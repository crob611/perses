// Copyright 2021 The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasource

import (
	"fmt"

	"github.com/perses/perses/internal/api/interface/v1/datasource"
	"github.com/perses/perses/internal/api/shared"
	databaseModel "github.com/perses/perses/internal/api/shared/database/model"
	"github.com/perses/perses/internal/api/shared/schemas"
	"github.com/perses/perses/internal/api/shared/validate"
	"github.com/perses/perses/pkg/model/api"
	v1 "github.com/perses/perses/pkg/model/api/v1"
	"github.com/sirupsen/logrus"
)

type service struct {
	datasource.Service
	dao datasource.DAO
	sch schemas.Schemas
}

func NewService(dao datasource.DAO, sch schemas.Schemas) datasource.Service {
	return &service{
		dao: dao,
		sch: sch,
	}
}

func (s *service) Create(entity api.Entity) (interface{}, error) {
	if datasourceObject, ok := entity.(*v1.Datasource); ok {
		return s.create(datasourceObject)
	}
	return nil, fmt.Errorf("%w: wrong entity format, attempting Datasource format, received '%T'", shared.BadRequestError, entity)
}

func (s *service) create(entity *v1.Datasource) (*v1.Datasource, error) {
	if err := s.validate(entity); err != nil {
		return nil, fmt.Errorf("%w: %s", shared.BadRequestError, err)
	}
	// Update the time contains in the entity
	entity.Metadata.CreateNow()
	if err := s.dao.Create(entity); err != nil {
		if databaseModel.IsKeyConflict(err) {
			logrus.Debugf("unable to create the Datasource %q. It already exits", entity.Metadata.Name)
			return nil, shared.ConflictError
		}
		logrus.WithError(err).Errorf("unable to perform the creation of the Datasource %q, something wrong with the database", entity.Metadata.Name)
		return nil, shared.InternalError
	}
	return entity, nil
}

func (s *service) Update(entity api.Entity, parameters shared.Parameters) (interface{}, error) {
	if DatasourceObject, ok := entity.(*v1.Datasource); ok {
		return s.update(DatasourceObject, parameters)
	}
	return nil, fmt.Errorf("%w: wrong entity format, attempting Datasource format, received '%T'", shared.BadRequestError, entity)
}

func (s *service) update(entity *v1.Datasource, parameters shared.Parameters) (*v1.Datasource, error) {
	if err := s.validate(entity); err != nil {
		return nil, fmt.Errorf("%w: %s", shared.BadRequestError, err)
	}
	if entity.Metadata.Name != parameters.Name {
		logrus.Debugf("name in Datasource %q and name from the http request %q don't match", entity.Metadata.Name, parameters.Name)
		return nil, fmt.Errorf("%w: metadata.name and the name in the http path request don't match", shared.BadRequestError)
	}
	if len(entity.Metadata.Project) == 0 {
		entity.Metadata.Project = parameters.Project
	} else if entity.Metadata.Project != parameters.Project {
		logrus.Debugf("project in datasource %q and project from the http request %q don't match", entity.Metadata.Project, parameters.Project)
		return nil, fmt.Errorf("%w: metadata.project and the project name in the http path request don't match", shared.BadRequestError)
	}
	// find the previous version of the Datasource
	oldEntity, err := s.Get(parameters)
	if err != nil {
		return nil, err
	}
	oldObject := oldEntity.(*v1.Datasource)
	entity.Metadata.Update(oldObject.Metadata)
	if err := s.dao.Update(entity); err != nil {
		logrus.WithError(err).Errorf("unable to perform the update of the Datasource %q, something wrong with the database", entity.Metadata.Name)
		return nil, shared.InternalError
	}
	return entity, nil
}

func (s *service) Delete(parameters shared.Parameters) error {
	if err := s.dao.Delete(parameters.Project, parameters.Name); err != nil {
		if databaseModel.IsKeyNotFound(err) {
			logrus.Debugf("unable to find the Datasource %q", parameters.Name)
			return shared.NotFoundError
		}
		logrus.WithError(err).Errorf("unable to delete the Datasource %q, something wrong with the database", parameters.Name)
		return shared.InternalError
	}
	return nil
}

func (s *service) Get(parameters shared.Parameters) (interface{}, error) {
	entity, err := s.dao.Get(parameters.Project, parameters.Name)
	if err != nil {
		if databaseModel.IsKeyNotFound(err) {
			logrus.Debugf("unable to find the Datasource %q", parameters.Name)
			return nil, shared.NotFoundError
		}
		logrus.WithError(err).Errorf("unable to find the previous version of the Datasource %q, something wrong with the database", parameters.Name)
		return nil, shared.InternalError
	}
	return entity, nil
}

func (s *service) List(q databaseModel.Query, _ shared.Parameters) (interface{}, error) {
	dtsList, err := s.dao.List(q)
	if err != nil {
		return nil, err
	}
	dtsQuery := q.(*datasource.Query)
	return v1.FilterDatasource(dtsQuery.Kind, dtsQuery.Default, dtsList), nil
}

func (s *service) validate(entity *v1.Datasource) error {
	var list []*v1.Datasource
	if entity.Spec.Default {
		var err error
		// return the full list of dts
		list, err = s.dao.List(&datasource.Query{Project: entity.Metadata.Project})
		if err != nil {
			logrus.WithError(err).Errorf("unable to get the list of the global datasource")
			return err
		}
	}
	return validate.Datasource(entity, list, s.sch)
}
